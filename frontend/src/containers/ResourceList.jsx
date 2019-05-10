import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import colors from 'colors.scss';
import analytics from 'common/analytics';
import Icon from 'components/Icon';
import DeleteInstance from './Dialogs/DeleteInstance';

import './ResourceList.scss';

class ResourceList extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  handleDeleteEnvironment(environment) {
    this.deleteInstanceDialog.getWrappedInstance().open(environment, 'environment');
  }

  @autobind
  handleDeleteDatabase(database) {
    this.deleteInstanceDialog.getWrappedInstance().open(database, 'database');
  }

  render() {
    const { environments, databases } = this.props;

    return (
      <>
        <div className="ResourceList">
          {environments.map(environment =>
            <div key={environment.id} className="ResourceListItem ItemEnvironment">
              <div className="overflow layout horizontal center">
                <div className="ItemIcon">
                  <Icon icon={environment.dev ? 'environmentOn' : 'environmentOff'} size="28"/>
                </div>
                <div className="ItemName ellipsis" 
                  title={environment.name}>
                  {environment.name}
                </div>
              </div>

              <div className="ItemEndpoints overflow">
                {environment.endpoints.map(url =>
                  <a className="ItemEndPoint layout horizontal center" 
                    key={`${environment.id}-${url}`}
                    href={url}
                    title={url}
                    onClick={() => analytics.track('Click Environment URL')}
                    rel="noreferrer noopener" 
                    target="_blank">
                    <div className="Url ellipsis">{url}</div>
                    <Icon icon="external" size="18" />
                  </a>
                )}
              </div>

              {environment.dev && 
                <div className="ItemActiveUser">
                  <div className="UserAtom layout horizontal center">
                    <div className="Avatar">
                      {!environment.dev.avatar && 
                        <Icon icon="logo" size="52" color={colors.navyDark} />
                      }
                      {environment.dev.avatar && 
                        <img src={environment.dev.avatar} width="22" />
                      }
                    </div>
                    <div className="Username">{environment.dev.githubID}</div>
                  </div>
                </div>
              }
              <div className="ItemActions">
                <div className="ActionButton" 
                  onClick={() => this.handleDeleteEnvironment(environment)}>
                  <Icon icon="trash" size="22" />
                </div>
              </div>
            </div>
          )}

          {databases.map(database =>
            <div key={database.name} className="ResourceListItem ItemDatabase">
              <div className="overflow layout horizontal center">
                <div className="ItemIcon">
                  <Icon icon="database" size="28"/>
                </div>
                <div className="ItemName ellipsis" 
                  title={database.name}>
                  {database.name}
                </div>
              </div>

              <div className="ItemEndpoints overflow ellipsis">
                <div className="ItemEndPoint">
                  <div className="Url">{database.endpoint}</div>
                </div>
              </div>

              <div className="ItemActions">
                <div className="ActionButton"
                  onClick={() => this.handleDeleteDatabase(database)}>
                  <Icon icon="trash" size="22" />
                </div>
              </div>
            </div>
          )}
        </div>

        <DeleteInstance ref={ref => this.deleteInstanceDialog = ref} />
      </>
    );
  }
}

ResourceList.propTypes = {
  dispatch: PropTypes.func.isRequired,
  environments: PropTypes.arrayOf(PropTypes.object).isRequired,
  databases: PropTypes.arrayOf(PropTypes.object).isRequired,
};

export default ReactRedux.connect(state => {
  return {
    environments: state.spaces.current.environments || [],
    databases: state.spaces.current.databases || []
  };
})(ResourceList);