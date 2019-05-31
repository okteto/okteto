import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import isWindows from 'is-windows';
import * as clipboard from 'clipboard-polyfill';
import autobind from 'autobind-decorator';

import analytics from 'common/analytics';
import ResourceList from 'containers/ResourceList';
import { notify } from 'components/Notification';
import Button from 'components/Button';
import Hint from 'components/Hint';
import Icon from 'components/Icon';
import NewMenu from 'components/NewMenu';
import SpaceHeader from './SpaceHeader';
import CreateDatabaseDialog from './Dialogs/CreateDatabase';
import CreateEnvironmentDialog from './Dialogs/CreateEnvironment';

import 'containers/Space.scss';

class Space extends Component {
  constructor(props) {
    super(props);

    this.state = {
      showNewMenu: false,
      showHintNew: true
    };
  }

  @autobind
  handleCreateDatabase() {
    this.createDatabaseDialog.getWrappedInstance().open();
  }

  @autobind
  handleCreateEnvironment() {
    this.createEnvironmentDialog.getWrappedInstance().open();
  }

  render() {
    const { space, environments, databases } = this.props;
    const isEmpty = environments.length === 0 && databases.length === 0;
    const isOnline = navigator.onLine;

    const NewButton = (props) => (
      <div className="NewButtonContainer">
        <div className="NewButton" onClick={() => this.setState({ showNewMenu: true })}>
          <Icon icon="plus" size="36" />
        </div>
        {this.state.showNewMenu && 
          <NewMenu
            position={props.menuPosition}
            onNewEnvironment={() => this.handleCreateEnvironment()}
            onNewDatabase={() => this.handleCreateDatabase()}
            onClose={() => this.setState({ showNewMenu: false })}
          />
        }
      </div>
    );

    const installCmdWin = 'wget https://downloads.okteto.com/cloud/cli/okteto-Windows-x86_64' + 
      ' -OutFile c:\\windows\\system32\\okteto.exe';
    const installCmdUnix = 'curl https://get.okteto.com -sSfL | sh';

    const HintContent = () => (
      <div className="HintContent layout vertical">
        <div className="step layout vertical">
          <p>
            <span className="number">1.</span> Install <strong>Okteto CLI</strong>:
          </p>
          <div className="layout horizontal">
            <code className="cli flex-auto">
              {!isWindows() &&
                <>{ installCmdUnix }</>
              }
              {isWindows() &&
                <>{ installCmdWin }</>
              }
            </code>
            <Button
              className="ClipboardButton"
              icon="clipboard"
              iconSize="24"
              onClick={() => {
                clipboard.writeText(isWindows() ? installCmdWin : installCmdUnix);
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
            <span className="number">2.</span> Login from the CLI:
          </p>
          <div className="layout horizontal">
            <code className="cli flex-auto">
              okteto login
            </code>
            <Button
              className="ClipboardButton"
              icon="clipboard"
              iconSize="24"
              onClick={() => {
                clipboard.writeText(`okteto login`);
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
            <span className="number">3.</span> From your <strong>local repository</strong> launch 
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
            You can now code <strong>in the cluster</strong> from your machine!
          </p>
        </div>

      </div>
    );

    const NewMenuContent = NewMenu.NewMenuContent(
      this.handleCreateEnvironment, this.handleCreateDatabase);

    return (
      <>
        <div className="Space">
          <SpaceHeader />

          {isEmpty && isOnline &&
            <div className="SpaceEmpty layout vertical center">
              <Icon icon="emptySpace" size="160" />
              <h2>Your namespace is empty.</h2>
              <div style={{ position: 'relative' }}>
                <Hint 
                  className="HintNew"
                  open={this.state.showHintNew}
                  width="300"
                  arrowPosition="center"
                  offsetY="24"
                  offsetX="0"
                  positionX="center"
                  positionY="bottom"
                  onTop={false}
                  hideCloseButton
                >
                  <h3>Create a new resource:</h3>
                  <NewMenuContent />
                </Hint>
              </div>
            </div>
          }

          {!isEmpty && isOnline &&
            <div className="SpaceContent">
              <ResourceList />
              <NewButton menuPosition="left" />
            </div>
          }

          {!isOnline && 
            <div className="SpaceOffline layout vertical center">
              <Icon icon="offline" size="140" /> 
              <h2>You are offline.</h2>
            </div>
          }
        </div>

        <CreateDatabaseDialog 
          ref={ref => this.createDatabaseDialog = ref}
          space={space}
        />
        
        <CreateEnvironmentDialog 
          ref={ref => this.createEnvironmentDialog = ref}
          space={space}
        >
          <HintContent />
        </CreateEnvironmentDialog>
      </>
    );  
  }
}

Space.propTypes = {
  dispatch: PropTypes.func.isRequired,
  space: PropTypes.object.isRequired,
  environments: PropTypes.arrayOf(PropTypes.object).isRequired,
  databases: PropTypes.arrayOf(PropTypes.object).isRequired
};

export default ReactRedux.connect(state => {
  return {
    space: state.spaces.current || {},
    environments: state.spaces.current.environments || [],
    databases: state.spaces.current.databases || []
  };
})(Space);