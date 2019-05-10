import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Button from 'components/Button';
import Input from 'components/Input';
import Icon from 'components/Icon';
import Avatar from 'components/Avatar';

import './SpaceInvite.scss';

class SpaceInvite extends Component {
  constructor(props) {
    super(props);

    this.state = {
      currentValue: '',
      members: props.members
    };
  }

  getMembers() {
    return this.state.members
  }

  @autobind
  handleAddMember() {
    const { members, currentValue } = this.state;
    const value = currentValue.trim();
    const compact = members => members.filter((v, i) => members.indexOf(v) === i);
    if (value) {
      this.setState({
        members: compact([value, ...members]),
        currentValue: ''
      });
    }
    this.inviteInput.focus();
  }

  @autobind
  handleRemoveMember(member) {
    this.setState({
      members: this.state.members.filter(m => m !== member)
    });
  }

  focus() {
    this.inviteInput.focus();
  }

  render() {
    return (
      <div className="SpaceInvite layout vertical">
        <div className="SpaceInviteInput layout horizontal center">
          <Input
            className="flex-auto"
            onChange={value => this.setState({ currentValue: value })}
            onKeyPress={event => {
              if (event.key == 'Enter') {
                this.handleAddMember();
              }
            }}
            ref={ref => this.inviteInput = ref}
            placeholder="username"
            theme="light"
            value={this.state.currentValue}
          />
          <Button onClick={this.handleAddMember} color="grey" solid>Add</Button>
        </div>
        <div className="SpaceInviteMembers layout vertical">
          {this.state.members.map(member => (
            <div 
              className="Member layout horizontal center" 
              key={member}
            >
              <Avatar username={member} size="18" />
              {member}

              <div className="flex-auto"></div>

              {this.props.owner !== member &&
                <div 
                  className="DeleteButton"
                  onClick={() => this.handleRemoveMember(member)}>
                  <Icon icon="trash" size="18" />
                </div>
              }
              {this.props.owner === member &&
                <div className="OwnerLabel">Owner</div>
              }
            </div>
          ))}
          {this.state.members.length === 0 && 
            <div className="EmptyMembers layout horizontal center">
              Add members to share your development space.
            </div>
          }
        </div>
      </div>
    );
  }
}

SpaceInvite.defaultProps = {
  members: [],
  owner: null
};

SpaceInvite.propTypes = {
  members: PropTypes.arrayOf(PropTypes.string),
  owner: PropTypes.string
};

export default SpaceInvite;