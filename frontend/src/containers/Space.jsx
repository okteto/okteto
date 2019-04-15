import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import isWindows from 'is-windows';
import * as clipboard from 'clipboard-polyfill';
import autobind from 'autobind-decorator';

import analytics from 'common/analytics';
import { refreshEnvironments } from 'actions/environments';
import { refreshDatabases } from 'actions/databases';
import { notify } from 'components/Notification';
import Button from 'components/Button';
import Hint from 'components/Hint';
import Icon from 'components/Icon';
import NewMenu from 'components/NewMenu';
import colors from 'colors.scss';
import Header from './Header';
import CreateDatabaseDialog from './Dialogs/CreateDatabase';
import CreateEnvironmentDialog from './Dialogs/CreateEnvironment';
import DeleteDialog from './Dialogs/DeleteDialog';

import 'containers/Space.scss';

const POLLING_INTERVAL = 10000;

class Space extends Component {
  constructor(props) {
    super(props);

    this.state = {
      showNewMenu: false
    };

    this.props.dispatch(refreshEnvironments());
    this.props.dispatch(refreshDatabases());
    this.poll = setInterval(this.handlePollEnvironments, POLLING_INTERVAL);
  }

  componentWillUnmount() {
    clearInterval(this.poll);
  }

  @autobind
  handlePollEnvironments() {
    // TODO: Should be merged into same graphql query.
    this.props.dispatch(refreshEnvironments());
    this.props.dispatch(refreshDatabases());
  }

  @autobind
  handleCreateDatabase() {
    this.createDatabaseDialog.getWrappedInstance().open();
  }

  @autobind
  handleCreateEnvironment() {
    this.createEnvironmentDialog.getWrappedInstance().open();
  }

  @autobind
  handleDeleteEnvironment(environment) {
    this.deleteDialog.getWrappedInstance().open(environment, 'environment');
  }

  @autobind
  handleDeleteDatabase(database) {
    this.deleteDialog.getWrappedInstance().open(database, 'database');
  }

  render() {
    const { environments, databases, user } = this.props;
    const environmentList = Object.values(environments);
    const databaseList = Object.values(databases);
    const isEmpty = environmentList.length === 0 && databaseList.length === 0;

    const HintContent = () => (
      <div className="HintContent layout vertical">
        <div className="step layout vertical">
          <p>
            <span className="number">1.</span> Install <strong>Okteto CLI</strong>:
          </p>
          <div className="layout horizontal">
            <code className="cli flex-auto">
              {!isWindows() &&
                <>curl https://okteto.com/get -sSfL | sh</>
              }
              {isWindows() &&
                <>wget https://downloads.okteto.com/cloud/cli/okteto-Windows-x86_64 -OutFile c:\windows\system32\okteto.exe</>
              }
            </code>
            <Button
              className="ClipboardButton"
              icon="clipboard"
              iconSize="24"
              onClick={() => {
                clipboard.writeText(`curl https://get.okteto.com -sSfL | sh`);
                notify('Copied to clipboard!');
                analytics.set('Copied Install Command');
                analytics.track('Copy Install Command');
              }}
              light
              frameless>
            </Button>
          </div>
        </div>

        <div className="step layout vertical">
          <p>
            <span className="number">2.</span> From your <strong>local repository</strong> launch 
            okteto:
          </p>
          <div className="layout horizontal">
            <code className="cli flex-auto">
              okteto up
            </code>
            <Button
              className="ClipboardButton"
              icon="clipboard"
              iconSize="24"
              onClick={() => {
                clipboard.writeText(`okteto up`);
                notify('Copied to clipboard!');
                analytics.set('Copied Okteto Command');
                analytics.track('Copy Okteto Command');
              }}
              light
              frameless>
            </Button>
          </div>
        </div>

        <div className="step layout vertical">
          <p>
            <span className="number">3.</span> Code in the cluster from your favorite IDE!
          </p>
          <div className="layout horizontal">
            <code className="cli cli-okteto flex-auto">
              yarn start<br/> 
              <span className="output">Running server at http://{user.githubID}.okteto.net</span>
            </code>
          </div>
        </div>

      </div>
    );

    return (
      <div className="Space layout vertical">
        <Header title={`${user.githubID}'s space`} />

        {isEmpty &&
          <div className="EmptySpace layout vertical center">
            <Icon icon="emptySpace" size="140" />
            <h2>Your space is empty.</h2>
            <div style={{
              position: 'relative'
            }}>
              <Hint 
                className="HintNew"
                open={true}
                width="532"
                arrowPosition="center"
                offsetY="24"
                offsetX="0"
                positionX="center"
                positionY="bottom"
                hideCloseButton
              >
                <h1>Start a new environment</h1>
                <HintContent />
              </Hint>
            </div>
          </div>
        }

        <div className="List layout vertical">
          {environmentList.map(environment =>
            <div key={environment.id} className="Item layout horizontal start">
              <div className="ItemIcon">
                <Icon icon="mirror" size="20"/>
              </div>
              <div className="ItemName ellipsis" 
                title={environment.name}>
                {environment.name}
              </div>
              <div className="ItemEndpoints layout vertical">
                {environment.endpoints.map(url =>
                  <a className="ellipsis layout horizontal center" 
                    key={`${environment.id}-${url}`}
                    href={url}
                    rel="noreferrer noopener" 
                    target="_blank">
                    {url}
                    <Icon icon="external" size="18" />
                  </a>
                )}
              </div>
              <div className="flex-auto" />
              <div className="ItemActions layout horizontal center">
                <div className="ActionButton" 
                  onClick={() => this.handleDeleteEnvironment(environment)}>
                  <Icon icon="delete" size="24" />
                </div>
              </div>
            </div>
          )}

          {databaseList.map(database =>
            <div key={database.name} className="Item layout horizontal start">
              <div className="ItemIcon">
                <Icon icon="database" size="20"/>
              </div>
              <div className="ItemName ellipsis" 
                title={database.name}>
                {database.name}
              </div>
              <div className="ItemEndpoints layout vertical">
                {database.endpoint}
              </div>
              <div className="flex-auto" />
              <div className="ItemActions layout horizontal center">
                <div className="ActionButton" 
                  onClick={() => this.handleDeleteDatabase(database)}>
                  <Icon icon="delete" size="24" />
                </div>
              </div>
            </div>
          )}
        </div>

        {!isEmpty && 
          <div className="ActionBar layout horizontal center">
            <div className="flex-auto"></div>
            <div className="NewButtonContainer">
              <div className="NewButton" onClick={() => this.setState({ showNewMenu: true })}>
                <Icon
                  className="NewButton" 
                  icon="plusCircle" 
                  size="36"
                  color={colors.green400}
                />
              </div>
              {this.state.showNewMenu && 
                <NewMenu
                  onNewEnvironment={() => this.handleCreateEnvironment()}
                  onNewDatabase={() => this.handleCreateDatabase()}
                  onClose={() => this.setState({ showNewMenu: false })}
                />
              }
            </div>
          </div>
        }

        <DeleteDialog ref={ref => this.deleteDialog = ref} />
        <CreateDatabaseDialog ref={ref => this.createDatabaseDialog = ref} />
        <CreateEnvironmentDialog ref={ref => this.createEnvironmentDialog = ref}>
          <HintContent />
        </CreateEnvironmentDialog>
      </div>
    );  
  }
}

Space.defaultProps = {
};

Space.propTypes = {
  dispatch: PropTypes.func,
  user: PropTypes.object.isRequired,
  environments: PropTypes.object.isRequired,
  databases: PropTypes.object.isRequired
};

export default ReactRedux.connect(state => {
  return {
    environments: state.environments.byId || {},
    databases: state.databases.byName || {},
    user: state.session.user
  };
})(Space);